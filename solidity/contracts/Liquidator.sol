// SPDX-License-Identifier: GPL-3.0
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/utils/Address.sol";

import "./interfaces/INFTVault.sol";

interface ICryptoPunks {
    function transferPunk(address to, uint256 punkIndex) external;
}

/// @title Liquidator escrow contract
/// @notice Liquidator contract that allows liquidator bots to liquidate positions without holding any PUSD/NFTs.
/// It's only meant to be used by DAO bots.
/// The liquidated NFTs are sent to the DAO
contract PunkLiquidator {
    using Address for address;

    INFTVault public immutable nftVault;
    ICryptoPunks public immutable cryptoPunks;
    IERC20 public immutable pusd;
    
    address public dao;

    constructor(
        INFTVault _nftVault,
        ICryptoPunks _cryptoPunks,
        IERC20 _pusd,
        address _dao
    ) {
        nftVault = _nftVault;
        cryptoPunks = _cryptoPunks;
        pusd = _pusd;
        dao = _dao;
    }

    modifier onlyDAO() {
        require(msg.sender == dao, "not dao");
        _;
    }

    /// @notice Allows any address to liquidate multiple positions at once.
    /// It assumes enough PUSD is in the contract.
    /// The liquidated punks are sent to the DAO
    /// @dev This function doesn't revert if one of the positions can't be liquidated.
    /// This is done to prevent situations in which multiple positions can't be liquidated
    /// because of one not liquidatable position.
    /// This function reverts when there's not enough PUSD in this contract
    /// @param _toLiquidate The positions to liquidate
    function liquidate(uint256[] memory _toLiquidate) external {
        uint256 balance = pusd.balanceOf(address(this));
        for (uint256 i = 0; i < _toLiquidate.length; i++) {
            uint256 nftIndex = _toLiquidate[i];
            INFTVault.PositionPreview memory position = nftVault.showPosition(
                nftIndex
            );
            //ignore not liquidatable position
            if (!position.liquidatable) continue;

            uint256 totalDebt = position.debtPrincipal + position.debtInterest;
            if (totalDebt > balance) continue;

            pusd.approve(address(nftVault), totalDebt);
            nftVault.liquidate(nftIndex);

            if (position.borrowType == INFTVault.BorrowType.NON_INSURANCE) {
                cryptoPunks.transferPunk(dao, nftIndex);
            }
        }
    }

    /// @notice Allows any address to claim NFTs from multiple expired insurance postions at once.
    /// The liquidated punks are sent to the DAO
    /// @dev This function doesn't revert if one of the NFTs isn't claimable yet. This is done to prevent
    /// situations in which multiple NFTs can't be claimed because of one not being claimable yet
    /// @param _toClaim The indexes of the NFTs to claim
    function claimExpiredInsuranceNFT(uint256[] memory _toClaim) external {
        for (uint256 i = 0; i < _toClaim.length; i++) {
            uint256 nftIndex = _toClaim[i];

            INFTVault.PositionPreview memory position = nftVault.showPosition(
                nftIndex
            );

            uint256 elapsed = block.timestamp - position.liquidatedAt;

            //ignore not claimable NFT
            if (elapsed < position.vaultSettings.insuraceRepurchaseTimeLimit)
                continue;

            nftVault.claimExpiredInsuranceNFT(nftIndex);
            cryptoPunks.transferPunk(dao, nftIndex);
        }
    }

    /// @notice Allows the DAO to perform multiple calls using this contract (recovering funds/NFTs stuck in this contract)
    /// @param targets The target addresses
    /// @param calldatas The data to pass in each call
    /// @param values The ETH value for each call
    function doCalls(address[] memory targets, bytes[] memory calldatas, uint256[] memory values) external payable onlyDAO {        
        for (uint256 i = 0; i < targets.length; i++) {
            targets[i].functionCallWithValue(calldatas[i], values[i]);
        }
    }

    /// @notice Allows the DAO to change the DAO address
    /// @param _dao The new DAO address
    function changeDAO(address _dao) external onlyDAO  {
        require(_dao != address(0), "invalid address");
        dao = _dao;
    }
}
